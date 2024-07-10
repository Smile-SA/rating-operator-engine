# rating-operator-engine

**DISCLAIMER** This component is part of the **rating-operator** application and is not designed to work as a standalone program.

**Rating-operator-engine** uses custom resources to configure its workers and trigger rating workloads, with a variable timeframe.
Each workers holds their own configuration, that can be updated at runtime, and query, rate and store data in **PostgreSQL** through the **rating-operator-api**.

### Configuration

The configuration, named *RatingRuleInstances*, can be sumed up as follow:

- **Name**
> The name of the rated metric. It is the name by which you'll be able to query your metric afterward.

- **Timeframe** (Base value of 60s)
> This parameters control the time between each data query, in seconds. Depending on the precision or reactivity need of a given metric, you can adapt this parameter to suit your need.
> Considering Prometheus expose datapoints for each seconds, "timeframe" imply the size of the metric set you receive.
> By using, for example, 3600s as the timeframe, you will receive, every hour, 3600 datapoints that will then be sumed into one.

- **Metric**
> A prometheus query expression (promQL) to be executed every "timeframe".
> It can be enhanced with variables, defined in the RatingRules configuration, and made available through Prometheus.
> More information on RatingRules can be read in the rating-operator ([Custom Resources](https://github.com/alterway/rating-operator/blob/master/documentation/CRD.md)) document.

Below an example of a RatingRuleInstance:

```yaml
apiVersion: rating.smile.fr/v1
kind: RatingRuleInstances
metadata:
  name: my-test-rule # The name of the RatingRuleInstances object
spec:
  metric: sum(kube_pod_container_resource_requests_cpu_cores) by (pod, namespace, node) # The prometheus query to execute
  name: pods_usage_cpu # The name of the metric
  timeframe: 60s # The time between each query
```

### Usage

For more informations on how to configure and use the component, please refer to the [rating-operator documentation](https://github.com/alterway/rating-operator/blob/master/README.md).
