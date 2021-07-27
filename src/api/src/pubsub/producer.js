
const { logger } = require('../../../lib/log')
const {PubSub} = require('@google-cloud/pubsub')

const pubSubClient = new PubSub()

function publishMessage(data) {
    const topicName = config.pubsub.topic
    const dataBuffer = Buffer.from(JSON.stringify(data));
    pubSubClient.topic(topicName).publish(dataBuffer, function (error, data) {
        if (error) {
            logger.error(`Received error while publishing: ${error.message}`)
        } else {
            logger.info(`Message ${data} published.`)
        }
    })
}

module.exports = publishMessage
