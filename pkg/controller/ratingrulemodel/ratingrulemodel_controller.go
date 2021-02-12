package ratingrulemodel

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	ratingv1 "github.com/rating-operator-engine/pkg/apis/rating/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// prometheusResultsArray is a wrapper struct to hold the loaded results from a query to Prometheus API.
type prometheusResultsArray struct {
	data []PrometheusResults
}

// The PrometheusResults struct is meant to be loaded with the result of a Prometheus API query.
type PrometheusResults struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values []interface{}     `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

//The Frame struc mimics the shape of a frame in database, to ease the work of rating-api.
type Frame struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`

	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`
	Metric    string `json:"metric"`

	// Labels should contains anything different from above variables
	Labels map[string]string `json:"labels"`

	// Quantity
	Value float64 `json:"quantity"`
}

// metricToFrame is here to interpolate the values in the metric to construct the frame to be sent.
// Default value are used if metric doesn't have the key.
// The data used is then removed from metric, as all leftovers are considered labels and will be used for later matching.
func metricToFrame(metricName string, metric map[string]string) Frame {
	defaultValue := "unspecified"
	frame := Frame{
		Metric: metricName,
	}

	if namespace, ok := metric["namespace"]; ok {
		frame.Namespace = namespace
		delete(metric, "namespace")
	} else {
		frame.Namespace = defaultValue
	}

	if node, ok := metric["node"]; ok {
		frame.Node = node
		delete(metric, "node")
	} else if instance, ok := metric["instance"]; ok {
		frame.Node = instance
		delete(metric, "instance")
	} else {
		frame.Node = defaultValue
	}

	if pod, ok := metric["pod"]; ok {
		frame.Pod = pod
		delete(metric, "pod")
	} else {
		frame.Pod = defaultValue
	}

	frame.Labels = metric
	return frame
}

// The ModelWatcher struct is the mechanism to handle each RatingRuleModels resources.
// stopChan and configChan are used for communication and configuration updates, in case of changes by the user.
type ModelWatcher struct {
	// Holds the configuration for the RatingRuleModel
	RatingRuleModel ratingv1.RatingRuleModelSpec

	Name   string
	Logger logr.Logger

	stopChan   chan struct{}
	configChan chan ratingv1.RatingRuleModelSpec

	UntreatedFrames PrometheusResults
	Frames          []Frame

	err         error
	client      *http.Client
	bearerToken string
}

// The ModelWatcher.timeframe function is a helper to generate a duration from the "timeframe" parameter of the RatingRuleModel.
func (mw *ModelWatcher) timeframe() int {
	timeframe := strings.ReplaceAll(mw.RatingRuleModel.Timeframe, "s", "")
	duration, err := strconv.Atoi(timeframe)
	if err != nil {
		mw.Logger.Error(err, "Unrecognized format for `timeframe`, using default (60s).")
		duration = 60
	}
	return duration
}

// The ModelWatcher.ticker function helps generating the time.Ticker object used by the ModelWatcher.
func (mw *ModelWatcher) ticker() *time.Ticker {
	duration := mw.timeframe()
	ticker := time.NewTicker(time.Second * time.Duration(duration))
	return ticker
}

// The ModelWatcher.query function get the data from the Prometheus API.
func (mw *ModelWatcher) query(ctx context.Context) *ModelWatcher {
	now := time.Now()
	end := now.Format(time.RFC3339)
	start := now.Add(time.Duration(-mw.timeframe()) * time.Second).Format(time.RFC3339)

	params := url.Values{
		"start": []string{start},
		"end":   []string{end},
		"query": []string{mw.RatingRuleModel.Metric},
		"step":  []string{"1s"},
	}

	endpoint := "/api/v1/query_range"
	encodedURL := fmt.Sprintf("%s%s?%s", os.Getenv("PROMETHEUS_URL"), endpoint, params.Encode())

	req, err := http.NewRequest("GET", encodedURL, nil)
	if err != nil {
		mw.Logger.Error(err, "error creating request")
		mw.err = err
		return mw
	}

	if mw.bearerToken != "" {
		req.Header.Add("Authorization", mw.bearerToken)
	}

	response, err := mw.client.Do(req)
	if err != nil {
		mw.Logger.Error(err, "error querying prometheus")
		mw.err = err
		return mw
	}
	defer response.Body.Close()

	if json.NewDecoder(response.Body).Decode(&mw.UntreatedFrames); err != nil {
		mw.Logger.Error(err, "error decoding response")
	}
	return mw
}

// The ModelWatcher.convert function convert the result of the Prometheus API query to exploitable data, AKA frames.
// It uses the reflect package to understand the PrometheusResults.
func (mw *ModelWatcher) convert(ctx context.Context) *ModelWatcher {
	mw.Frames = make([]Frame, 0)
	if mw.err != nil {
		mw.Logger.Info("error in queries, skipping convert step")
		return mw
	}

	for _, result := range mw.UntreatedFrames.Data.Result {
		frame := metricToFrame(mw.RatingRuleModel.Name, result.Metric)
		values := []float64{}
		for _, v := range result.Values {
			switch reflect.TypeOf(v).Kind() {
			case reflect.Slice:
				s := reflect.ValueOf(v)
				for idx := 0; idx < s.Len(); idx++ {
					value := s.Index(idx).Interface()
					switch reflect.TypeOf(value).Kind() {
					case reflect.Float64:
						if frame.Start == 0 {
							frame.Start = value.(float64)
						}
						frame.End = value.(float64)
					case reflect.String:
						converted, err := strconv.ParseFloat(value.(string), 64)
						if err != nil {
							mw.Logger.Error(err, "Couldn't convert string to float")
						}
						values = append(values, converted)
					}
				}
			}
		}

		frame.Value = average(values)
		mw.Frames = append(mw.Frames, frame)
	}
	mw.UntreatedFrames = PrometheusResults{}
	return mw
}

// average gives the average of the given float array.
func average(xs []float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += v
	}
	return total / float64(len(xs))
}

// The ModelWatcher.send function takes the frames stored and transmit them to the rating-api.
// These frames will be written to the storage by the api.
func (mw *ModelWatcher) send(ctx context.Context) error {
	if mw.err != nil {
		mw.Logger.Info("error in queries, skipping send step")
		return mw.err
	}
	encoded := new(bytes.Buffer)
	json.NewEncoder(encoded).Encode(mw.Frames)

	if frameCount := len(mw.Frames); frameCount > 0 {
		mw.Logger.Info(fmt.Sprintf("%d frames ready", frameCount))
	} else {
		return nil
	}

	endpoint := "/models/frames/add"
	addr := fmt.Sprintf("%s%s", os.Getenv("RATING_API_URL"), endpoint)
	response, err := http.PostForm(addr, url.Values{
		"rated_frames": []string{encoded.String()},
		"metric":       []string{mw.RatingRuleModel.Name},
		"last_insert":  []string{time.Now().Format(time.RFC3339)},
		"token":        []string{os.Getenv("RATING_ADMIN_API_KEY")},
	})

	if err != nil {
		mw.Logger.Error(err, "Couldn't send the payload")
		return err
	}
	mw.Logger.Info("Frames sent")
	defer response.Body.Close()

	mw.Frames = nil
	return nil
}

// The ModelWatcher.exit function is used to remove oneself from the watchers array
// It is called by the ModelWatcher.watch function right after the context deletion, at exit time.
func (mw *ModelWatcher) exit() {
	lock.Lock()
	delete(watchers, mw.Name)
	lock.Unlock()
	mw.Logger.Info("ModelWatcher exited")
}

// The ModelWatcher.watch function is the core logic of this operator.
// It uses a ticker to control the for loop, and maintain an up to date configuration through channels.
// Every tick, it generate a goroutine to execute the rating.
// The go keyword here is not mandatory, but helps handling edge cases (rating time > ticking time).
// The whole function is handled with a context, that is transmitted to the rating goroutine.
func (mw *ModelWatcher) watch(ctx context.Context) {
	parent, cancel := context.WithCancel(ctx)
	mw.Logger = log.WithValues("ModelWatcher.Name", mw.Name)
	mw.Logger.Info("ModelWatcher initialized")
	defer mw.exit()

	ticker := mw.ticker()
	lock.Lock()
	watchers[mw.Name] = mw
	lock.Unlock()
	for {
		select {
		case config := <-mw.configChan:
			mw.Logger.Info("ModelWatcher reconfigured")
			mw.RatingRuleModel = config
			ticker = mw.ticker()
		case <-mw.stopChan:
			cancel()
			return
		case <-ticker.C:
			go mw.query(parent).convert(parent).send(parent)
		}
	}
}

// The watchers variable is here to hold all the ModelWatcher pointers, to ease manipulation.
// It is coupled with a global mutex to protect from concurrent map access.
var watchers = make(map[string]*ModelWatcher)
var lock = sync.RWMutex{}

// The main logging object, that is used to generate sub-loggers for each RatingRuleModels.
var log = logf.Log.WithName("controller_ratingrulemodel")

// Add creates a new RatingRuleModel Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRatingRuleModel{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ratingrulemodel-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RatingRuleModel
	err = c.Watch(&source.Kind{Type: &ratingv1.RatingRuleModel{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// bearerToken helps registering authorization directly in the ModelWatcher.
func bearerToken(logger logr.Logger) (auth string) {
	auth = ""
	if os.Getenv("AUTH") == "true" {
		token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			logger.Error(err, "error reading token file")
			return
		}
		auth = "Bearer " + string(token)
		logger.Info("registered bearer token")
	}
	return
}

// createClient generate a http.Client with users credentials, or a minimal configuration.
func createClient(logger logr.Logger) *http.Client {
	client := &http.Client{}
	if os.Getenv("AUTH") == "true" {
		cert, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt")
		if err != nil {
			logger.Error(err, "error reading certificates")
			return client
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cert)

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
		}
	}
	return client
}

// blank assignment to verify that ReconcileRatingRuleModel implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRatingRuleModel{}

// ReconcileRatingRuleModel reconciles a RatingRuleModel object
type ReconcileRatingRuleModel struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RatingRuleModel object and makes changes based on the state read
// and what is in the RatingRuleModel.Spec
func (r *ReconcileRatingRuleModel) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RatingRuleModel")

	ctx := context.Background()
	// Fetch the RatingRuleModel instance
	instance := &ratingv1.RatingRuleModel{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			watchers[request.Name].stopChan <- struct{}{}
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	lock.Lock()
	watcher, exist := watchers[request.Name]
	lock.Unlock()

	// Update or create the ModelWatcher
	if exist {
		watcherConfig := ratingv1.RatingRuleModelSpec{
			Timeframe: instance.Spec.Timeframe,
			Metric:    instance.Spec.Metric,
			Name:      instance.Spec.Name,
		}
		watcher.configChan <- watcherConfig
	} else {
		watcher := &ModelWatcher{
			RatingRuleModel: ratingv1.RatingRuleModelSpec{
				Timeframe: instance.Spec.Timeframe,
				Metric:    instance.Spec.Metric,
				Name:      instance.Spec.Name,
			},
			Name:        request.Name,
			stopChan:    make(chan struct{}),
			configChan:  make(chan ratingv1.RatingRuleModelSpec),
			client:      createClient(reqLogger),
			bearerToken: bearerToken(reqLogger),
		}

		// The watcher is now configured and ready to run.
		// After start comes the registration of the ModelWatcher pointer to the global watchers array.
		go watcher.watch(ctx)
	}

	return reconcile.Result{}, nil
}
