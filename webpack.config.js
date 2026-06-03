// @ts-check
const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');

/** @type {(env: any, argv: any) => import('webpack').Configuration} */
module.exports = (_env, _argv) => ({
  context: path.resolve(__dirname, 'src'),
  devtool: 'source-map',
  entry: {
    module: './module.ts',
  },
  output: {
    clean: true,
    filename: '[name].js',
    path: path.resolve(__dirname, 'dist'),
    libraryTarget: 'amd',
    publicPath: 'public/plugins/dubsector-ciscomeraki-datasource/',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js', '.jsx'],
  },
  module: {
    rules: [
      {
        test: /\.[tj]sx?$/,
        exclude: /node_modules/,
        use: {
          loader: 'swc-loader',
          options: {
            jsc: {
              target: 'es2018',
              parser: {
                syntax: 'typescript',
                tsx: true,
                decorators: false,
                dynamicImport: true,
              },
            },
          },
        },
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
    ],
  },
  externals: [
    'lodash',
    'jquery',
    'moment',
    'slate',
    'emotion',
    '@emotion/react',
    '@emotion/css',
    'prismjs',
    'react',
    'react-dom',
    'react-redux',
    'redux',
    'rxjs',
    'rxjs/operators',
    'd3',
    'angular',
    '@grafana/ui',
    '@grafana/runtime',
    '@grafana/data',
    '@grafana/schema',
  ],
  plugins: [
    new CopyWebpackPlugin({
      patterns: [
        { from: 'plugin.json', to: '.' },
        {
          from: path.resolve(__dirname, 'dashboards'),
          to: 'dashboards',
          noErrorOnMissing: true,
        },
        {
          from: path.resolve(__dirname, 'img'),
          to: 'img',
          noErrorOnMissing: true,
        },
      ],
    }),
  ],
});
