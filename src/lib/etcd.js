const { Etcd3 } = require('etcd3')

client = new Etcd3({
    hosts: process.env.ETCD_HOSTS || config.etcd.hosts,
    auth: {
        username: process.env.ETCD_USERNAME || config.etcd.username,
        password: process.env.ETCD_PASSWORD || config.etcd.password,
    },
})

module.exports = client
