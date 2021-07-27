const config = global.config = require('../config/index')()
const { logger } = require('../../lib/log')
const stat = require('../src/api/stat')
const {PubSub} = require('@google-cloud/pubsub')

const pubSubClient = new PubSub()

function listenForMessages() {
    const subscriptionName = config.pubsub.subscription
    const subscription = pubSubClient.subscription(subscriptionName)

    subscription.on('message', message => {
        const msg = JSON.parse(message.data)
        switch (msg.key) {
            case 'request': {
                stat.request(msg.message)
            }
            case 'connections': {
                //统计当前连接数
            }
            default:
                break
        }
        logger.info(message.data)

        // "Ack" (acknowledge receipt of) the message
        message.ack()
    })

    subscription.on('error', error => {
        logger.error('Received error:', error)
    })
}

listenForMessages()
