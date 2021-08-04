const Koa = require('koa')
const koaBody = require('koa-body')
const router = require('./router')
const {
    logger,
    accessLogger
} = require('../lib/log')
const Result = require('../lib/result')
const config = global.config = require('./config/index')()
const app = new Koa()
const WebSocketServer = require('ws').Server;
const {
    accept
} = require('./src/routers/ws')
const crypto = require("crypto");
const Messengers = require("./src/api/messenger")
const publishMessage = require("./src/pubsub/producer")

app.keys = config.keys
app
    .use(koaBody({
        multipart: true
    }))
    .use(accessLogger())
    .use(async (ctx, next) => {
        const start = ctx[Symbol.for('request-received.startTime')] ? ctx[Symbol.for('request-received.startTime')].getTime() : Date.now()
        await next()
        logger.info(ctx.method, ctx.originalUrl, ctx.request.body, ctx.response.status || 404, ctx.response.length, 'byte', (Date.now() - start), 'ms')
    })
    .use(async (ctx, next) => {
        return next().catch((error) => {
            let code = 500
            let message = 'unknown error'
            let data = ''
            logger.error(error)
            if (error instanceof Result) {
                code = error.code
                message = error.message
            }
            ctx.body = {
                code,
                message,
                data
            }
        })
    })
    .use((ctx, next) => {
        ctx.set('Access-Control-Allow-Origin', '*')
        ctx.set('Content-Type', 'application/json')
        ctx.set('Access-Control-Expose-Headers', 'Access-Control-Allow-Origin')
        ctx.set('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept, Authorization')
        ctx.set('Access-Control-Allow-Methods', 'PUT, POST, GET, DELETE, PATCH, OPTIONS')
        ctx.set('Access-Control-Allow-Credentials', true)
        return next()
    })
    .use(router())

app.on('error', error => {
    logger.error(error)
})

global.messengers = new Messengers()
global.conWs = {}

let server = app.listen(config.port)
let wss = new WebSocketServer({
    server: server,
    clientTracking: true
});
wss.on('connection', function (ws, request) {
    logger.info('wss connection ', wss.clients.size)
    publishMessage({
        'key': 'connections',
        'message': {
            protocol: 'websocket',
            pid: process.pid, //考虑多进程
            connections: wss.clients.size
        }
    });
    let id = crypto.randomBytes(16).toString('hex');
    accept(id, ws, request)
})


wss.on('error', (error) => {
    logger.error('wss error', error)
})

wss.on('close', () => {
    logger.info('wss close')
})

process.on('unhandledRejection', (reason, p) => {
    logger.error('Unhandled Rejection at:', p, 'reason:', reason);
});
process.on('uncaughtException', function (e) {
    logger.error('uncaughtException', e)
})
logger.info(config.name, ' started listen on ', config.port)


// watch etcd config
const extend = require('extend')
const etcdConfig = require('./config/etcd')
etcdConfig(data => {
    extend(config, JSON.parse(data))
    console.log('changed', config)
    global.messengers.changed()
}).then(data => {
    extend(config, JSON.parse(data))
    console.log('loaded', config)
    global.messengers.changed()
}).catch(err => {
    throw JSON.stringify({ text: `Load Etcd Config Error：${err}` })
})
