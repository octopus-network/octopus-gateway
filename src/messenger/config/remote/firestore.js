const firestore = require("../../../lib/firestore")

const key = 'octopus-gateway/messenger'

module.exports = function firestoreConfig(callback) {
    const docRef = firestore.doc(key)
    docRef.onSnapshot(snapshot => {
        if (snapshot.exists) {
            const data = snapshot.data()
            const keys = Object.keys(data)
            if (keys.includes('config.json')) {
                callback(data) // config.json & chain1 & chain2
            }
        }
    }, err => {
        console.log(`Error: ${err}`)
    })
}
