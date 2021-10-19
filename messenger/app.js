const Koa = require('koa')
const koaBody = require('koa-body')
const {
    logger,
    accessLogger
} = require('../lib/log')
const Result = require('../lib/result')
global.config = require('./config/index')()
const app = new Koa()
const WebSocketServer = require('ws').Server;
const crypto = require("crypto");
const Router = require('./src/router');

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

app.on('error', error => {
    logger.error(error)
})

let router = new Router();
let server = app.listen(config.port)
let wss = new WebSocketServer({
    server: server,
    clientTracking: true
});
wss.on('connection', function (ws, request) {
    logger.info('wss connection ', wss.clients.size)
    
    let id = crypto.randomBytes(16).toString('hex');
    router.accept(id, ws)
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


// watch remote config
const path = require('path')
const extend = require('extend')

// -- etcd --
// const etcdConfig = require('./config/remote/etcd')
// etcdConfig((key, val) => {
//     if (key.endsWith('config.json')) {
//         extend(true, config, JSON.parse(val))
//         console.log('changed', config)
//         router.changed()
//     } else {
//         const chain = path.basename(key, '.json')
//         if (!config.chain[chain]) {
//             config.chain[chain] = {}
//         }
//         extend(true, config.chain[chain], JSON.parse(val))
//         console.log('changed', config)
//     }
// }).then(data => {
//     for (const key in data) {
//         if (key.endsWith('config.json')) {
//             extend(true, config, JSON.parse(data[key]))
//         } else {
//             const chain = path.basename(key, '.json')
//             if (!config.chain[chain]) {
//                 config.chain[chain] = {}
//             }
//             extend(true, config.chain[chain], JSON.parse(data[key]))
//         }
//     }
//     console.log('loaded', config)
//     router.changed()
// }).catch(err => {
//     throw JSON.stringify({ text: `Load Etcd Config Error：${err}` })
// })

// -- firestore --
const firestoreConfig = require('./config/remote/firestore')
firestoreConfig(data => {
    for (const key in data) {
        if (key == 'config.json') {
            extend(true, config, JSON.parse(data[key]))
        }
        else {
            const chain = path.basename(key, '.json')
            if (!config.chain[chain]) {
                config.chain[chain] = {}
            }
            extend(true, config.chain[chain], JSON.parse(data[key]))
        }
    }
    console.log('config:', config)
    router.changed()
})
