const fs = require('fs');
const path = require('path');
const webpack = require('webpack');
const confJSON = require('./conf.json');
const isProduction = process.env.NODE_ENV === 'production'; 
const { VueLoaderPlugin } = require('vue-loader');
const SWPrecacheWebpackPlugin = require('sw-precache-webpack-plugin');
const BundleAnalyzerPlugin = require('webpack-bundle-analyzer').BundleAnalyzerPlugin;
const CleanWebpackPlugin = require('clean-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const MiniCssExtractPlugin = require("mini-css-extract-plugin");

module.exports = {
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'resources/js'),
      'fortawesomeDir': path.resolve(__dirname, 'node_modules/@fortawesome')
    }
  },
  entry: {
    bundle: ['@/app.js', './resources/sass/app.scss']
  },
  output: {
    path: path.resolve(__dirname, 'public'),
    filename: '[name].[chunkhash].js',
    chunkFilename: 'js/[name].[chunkhash].js',
    publicPath: '/'
  },
  module: {
    rules: [
      {
        test: /\.(html)$/,
        use: {
          loader: 'html-loader',
          options: {
            attrs: [':data-src']
          }
        }
      },
      {
        test: /\.vue$/,
        use: 'vue-loader'
      },
      {
        test: /\.hbs$/,
        loader: 'handlebars-loader'
      },
      {
        test: /\.pug$/,
        oneOf: [
          // this applies to `<template lang="pug">` in Vue components
          {
            resourceQuery: /^\?vue/,
            use: ['pug-plain-loader']
          },
          // this applies to pug imports inside JavaScript
          {
            use: ['raw-loader', 'pug-plain-loader']
          }
        ]
      },
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: {
          loader: 'babel-loader',
        }
      },
      {
        test: /\.(woff(2)|woff|eot|ttf|png|jpg|gif|svg)$/,
        loader: 'file-loader',
        options: {
          name: '[name].[ext]?[hash]'
        }
      },
      {
        test: /\.(sa|sc)ss$/,
        use: [{
          loader: "vue-style-loader"
        }, {
          loader: isProduction ?  MiniCssExtractPlugin.loader : "style-loader"
        },{
          loader: "css-loader"
        }, {
          loader: "sass-loader",
        }]
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader']
      }
    ]
  },
  plugins: [
    new CleanWebpackPlugin('public/'),
    new webpack.ContextReplacementPlugin(/moment[\/\\]locale$/, /de|en|id/),
    new SWPrecacheWebpackPlugin({
      cacheId: 'tanibox',
      filename: 'service-worker.js',
      staticFileGlobs: ['public/**/*.{js,html,css}'],
      minify: true,
      stripPrefix: 'public/'
    }),
    new VueLoaderPlugin(),
    new BundleAnalyzerPlugin({
      analyzerMode: 'static',
      reportFilename: 'report.html',
      defaultSizes: 'parsed',
      openAnalyzer: false,
      generateStatsFile: false,
      statsFilename: 'stats.json',
      statsOptions: null,
      logLevel: 'info'
    }),
    new HtmlWebpackPlugin({
      template: 'resources/index.hbs',
      inject: true,
      chunkSortMode: 'dependency',
      serviceWorkerLoader: `<script type="text/javascript">${fs.readFileSync(path.join(__dirname, isProduction ? './resources/js/service-worker-prod.js' : './resources/js/service-worker-dev.js'), 'utf-8')}</script>`
    }),
    new MiniCssExtractPlugin({
      // Options similar to the same options in webpackOptions.output
      // both options are optional
      filename: isProduction ? '[name].css' : '[name].[hash].css',
      chunkFilename: isProduction ? '[id].css' : '[id].[hash].css',
    }),
    new webpack.DefinePlugin({
      'process.env': {
        CLIENT_ID: JSON.stringify(confJSON.client_id)
      },
    }),
    new webpack.ProvidePlugin({
      $: 'jquery',
      jQuery: 'jquery',
      Popper: ['popper.js', 'default']
    })
  ]
}

// test specific setups
if (process.env.NODE_ENV === 'test') {
  module.exports.externals = [require('webpack-node-externals')()]
  module.exports.devtool = 'inline-cheap-module-source-map'
}
