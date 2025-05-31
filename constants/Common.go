package constants

const (
	JobProducerWorkerCount      = 5 // This can remain if still relevant for producer side
	DefaultKafkaBootstrapServers = "localhost:29092"
	DefaultJobTopic              = "JobQueue"
	DefaultConsumerGroupID       = "gsched-default-group"
)
