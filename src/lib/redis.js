const fs = require('fs')
const { logger } = require('./log')
var Redis = require('ioredis')

client = new Redis({
    port: config.redis.port,
    host: config.redis.host,
    password: process.env.REDIS_PASSWORD || config.redis.password,
    tls: {
        ca: process.env.REDIS_TLS_CRT || fs.readFileSync(config.redis.cert),
    },
})
client.on("error", function (error) {
    logger.error('redis error', error)
})

module.exports = client