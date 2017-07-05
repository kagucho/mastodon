import loadPolyfills from '../mastodon/load_polyfills';

loadPolyfills().then(() => {
  debugger;
  //require('../mastodon/main').default(require('../mastodon/containers/new_status'));
}).catch(e => {
  console.error(e);
});
