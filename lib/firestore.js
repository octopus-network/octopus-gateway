const { Firestore } = require('@google-cloud/firestore');

// GOOGLE_APPLICATION_CREDENTIALS
client = new Firestore();

module.exports = client
