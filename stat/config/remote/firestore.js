const firestore = require("../../../lib/firestore")

const key = 'octopus-gateway/stat'

module.exports = function firestoreConfig(callback) {
    const docRef = firestore.doc(key)
    docRef.onSnapshot(snapshot => {
        if (snapshot.exists) {
            const config = snapshot.data()['config.json']
            if (config) {
                callback(config) // json string
            }
        }
    }, err => {
        console.log(`Error: ${err}`)
    })
}