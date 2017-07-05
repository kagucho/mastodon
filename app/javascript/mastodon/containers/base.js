import React from 'react';
import { Provider } from 'react-redux';
import configureStore from '../store/configureStore';
import { hydrateStore } from '../actions/store';
import { IntlProvider, addLocaleData } from 'react-intl';
import { getLocale } from '../locales';
const { localeData, messages } = getLocale();
addLocaleData(localeData);

export const store = configureStore();
const hydrateAction = hydrateStore(JSON.parse(document.getElementById('initial-state').textContent));
store.dispatch(hydrateAction);

export default class Base extends React.PureComponent {
  render () {
    const { children, locale } = this.props;

    return (
      <IntlProvider locale={locale} messages={messages}>
        <Provider store={store}>
          {children}
        </Provider>
      </IntlProvider>
    );
  }
}
