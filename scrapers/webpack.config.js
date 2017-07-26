const webpack = require('webpack');

module.exports = {
	target: 'node',
	module: {
		loaders: [
			{ test: /\.js$/, exclude: /node_modules/, loader: 'babel-loader', query: { presets: ['es2015'] } }
		]
	},
	plugins: [
		new webpack.optimize.UglifyJsPlugin({ test: /^/ }),
		new webpack.BannerPlugin({ banner: '#!/usr/bin/env node', raw: true })
	]
}
