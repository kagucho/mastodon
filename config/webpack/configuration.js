// Common configuration for webpacker loaded from config/webpacker.yml

const { join, resolve } = require('path');
const { env } = require('process');
const { safeLoad } = require('js-yaml');
const { readFileSync } = require('fs');

const configPath = resolve('config', 'webpacker.yml');
const loadersDir = join(__dirname, 'loaders');
const settings = safeLoad(readFileSync(configPath), 'utf8')[env.NODE_ENV];

const themePath = resolve('config', 'themes.yml');
const themes = safeLoad(readFileSync(themePath), 'utf8');

function removeOuterSlashes(string) {
  return string.replace(/^\/*/, '').replace(/\/*$/, '');
}

function formatPublicPath(host = '') {
  let formattedHost = removeOuterSlashes(host);
  if (formattedHost && !/^http/i.test(formattedHost)) {
    formattedHost = `//${formattedHost}`;
  }
  return `${formattedHost}/`;
}

const output = {
  path: resolve('public'),
  publicPath: formatPublicPath(env.CDN_HOST),
};

module.exports = {
  settings,
  themes,
  env: {
    CDN_HOST: env.CDN_HOST,
    NODE_ENV: env.NODE_ENV,
  },
  loadersDir,
  output,
};
