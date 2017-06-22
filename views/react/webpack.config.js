const path = require('path');

module.exports = {
  context: __dirname,
  entry: {
    reflagvsflag: __dirname + '/lib/js/src/reflagvsflag.js',
  },
  output: {
    path: __dirname + '/dist/',
    filename: '[name].js'
  },
  resolve: {
    extensions: ['.js', '.json']
  }
};
