const etcd = require("../../lib/etcd")

const prefix = 'octopus/gateway/messenger/'
// const key = 'octopus/gateway/messenger/config.json'

module.exports = async function etcdConfig(callback) {
    etcd.watch()
        .prefix(prefix)
        .create()
        .then(watcher => {
            watcher
                .on('put', res => {
                    console.log(res.key.toString(), 'changed to:', res.value.toString())
                    callback(res.key.toString(), res.value.toString())
                })
        });

    return await client.getAll().prefix(prefix).strings()
}
