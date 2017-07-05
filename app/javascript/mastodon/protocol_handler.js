import { store } from './containers/base';

export const register = () => {
  if (!window.navigator.registerProtocolHandler) {
    return;
  }

  const title = `${store.getState().getIn(['meta', 'domain'])} (Mastodon)`;

  if (process.env.NODE_ENV === 'development') {
    window.navigator.registerProtocolHandler('web+activity+http', '/process_activity?url=%s', title);
  }

  window.navigator.registerProtocolHandler('web+activity+https', '/process_activity?url=%s', title);
};
