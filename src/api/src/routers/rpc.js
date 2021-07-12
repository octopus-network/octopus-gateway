const {
    logger
} = require('../../../lib/log')
const superagent = require('superagent')
const {
    isUnsafe
} = require('../../../lib/helper/check')
const CODE = require('../../../lib/helper/code')
const crypto = require("crypto");
const { toJSON } = require("../../../lib/helper/assist")

let api = async (ctx, next) => {
    let chain = ctx.request.params.chain.toLowerCase()
    let pid = ctx.request.params.pid
    let req = ctx.request.body

    let check = await superagent.get(config.statServer + '/limit/' + chain + '/' + pid).query({})
    if (0 == check.body.code) {
        if (isUnsafe(req)) {
            ctx.response.body = JSON.stringify({
                "jsonrpc": req.jsonrpc,
                "error": CODE.UNSAFE_METHOD,
                "id": req.id
            })
            return next()
        }

        let start = (new Date()).getTime()
        let end = start
        let id = crypto.randomBytes(16).toString('hex');
        ctx.response.body = await (function () {
            return new Promise((resolve, reject) => {
                global.messengers.httpClient(id, chain, req, async (resp) => {
                    end = (new Date()).getTime()
                    resolve(resp)
                })
            })
        })()
    } else {
        logger.error(chain, pid, check.body)
        ctx.response.body = check.body
        return next()
    }

    return next()
}

let healthz = async (ctx, next) => {
    ctx.response.body = JSON.stringify(CODE.SUCCESS)
    return next()
}

module.exports = {
    'POST /:chain/:pid([a-z0-9]{32})': api,
    'GET /healthz': healthz
}