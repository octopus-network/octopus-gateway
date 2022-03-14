const { logger } = require('./log')
var Redis = require('ioredis')

client = new Redis({
    host: process.env.REDIS_HOST || config.redis.host,
    port: process.env.REDIS_PORT || config.redis.port,
    password: process.env.REDIS_PASSWORD || config.redis.password,
    tls: {
        ca: process.env.REDIS_TLS_CRT || config.redis.cert,
    },
})
client.on("error", function (error) {
    logger.error('redis error', error)
})

module.exports = client