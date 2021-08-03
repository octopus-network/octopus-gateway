const etcd = require("../../lib/etcd")

const key = 'octopus/gateway/stat/config.json'

module.exports = async function etcdConfig(callback) {
    etcd.watch()
        .key(key)
        .create()
        .then(watcher => {
            watcher
                .on('put', res => {
                    console.log(key, 'changed to:', res.value.toString())
                    callback(res.value.toString())
                })
        });

    return await client.get(key).string()
}
