const etcd = require("../../lib/etcd")

const prefix = 'octopus/gateway/messenger/'
// const key = 'octopus/gateway/messenger/config.json'
// const key = 'octopus/gateway/messenger/chains/aaaa.json'
// const key = 'octopus/gateway/messenger/chains/bbbb.json'

module.exports = async function etcdConfig(callback) {
    etcd.watch()
        .prefix(prefix)
        .create()
        .then(watcher => {
            watcher
                .on('put', res => {
                    // console.log(res.key.toString(), 'set to:', res.value.toString())
                    callback(res.key.toString(), res.value.toString())
                })
        });

    return await client.getAll().prefix(prefix).strings()
}
