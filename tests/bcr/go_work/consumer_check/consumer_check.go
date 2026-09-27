package consumer_check

import "example.org/consumer"

// Depending on consumer is what triggers fetching and building it for real.
var UsesConsumer = consumer.ConsumedRepoConfigRlocationPath
