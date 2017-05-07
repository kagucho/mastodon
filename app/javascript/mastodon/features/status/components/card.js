import React from 'react';
import ImmutablePropTypes from 'react-immutable-proptypes';
import PropTypes from 'prop-types';
import punycode from 'punycode';
import { defineMessages, injectIntl } from 'react-intl';

const IDNA_PREFIX = 'xn--';
const messages = defineMessages(
  {video: {id: 'status.video', defaultMessage: 'Video'}});

const decodeIDNA = domain => {
  return domain
    .split('.')
    .map(part => part.indexOf(IDNA_PREFIX) === 0 ? punycode.decode(part.slice(IDNA_PREFIX.length)) : part)
    .join('.');
};

const getHostname = url => {
  const parser = document.createElement('a');
  parser.href = url;
  return parser.hostname;
};

@injectIntl
export default class Card extends React.PureComponent {

  static propTypes = {
    card: ImmutablePropTypes.map,
    intl: PropTypes.object.isRequired,
    statusId: PropTypes.number.isRequired,
  };

  renderLink () {
    const { card } = this.props;

    let image    = '';
    let provider = card.get('provider_name');

    if (card.get('image')) {
      image = (
        <div className='status-card__image'>
          <img src={card.get('image')} alt={card.get('title')} className='status-card__image-image' />
        </div>
      );
    }

    if (provider.length < 1) {
      provider = decodeIDNA(getHostname(card.get('url')));
    }

    return (
      <a href={card.get('url')} className='status-card' target='_blank' rel='noopener'>
        {image}

        <div className='status-card__content'>
          <strong className='status-card__title' title={card.get('title')}>{card.get('title')}</strong>
          <p className='status-card__description'>{(card.get('description') || '').substring(0, 50)}</p>
          <span className='status-card__host'>{provider}</span>
        </div>
      </a>
    );
  }

  renderPhoto () {
    const { card } = this.props;

    return (
      <a href={card.get('url')} className='status-card-photo' target='_blank' rel='noopener'>
        <img src={card.get('url')} alt={card.get('title')}
             width={card.get('width')} height={card.get('height')}
             nonce={document.getElementById("img-nonce").textContent} />
      </a>
    );
  }

  renderVideo () {
    const { intl, statusId } = this.props;

    return (
      <div className='status-card-video'>
        <iframe src={`/api/v1/statuses/${statusId}/card_html`} title={intl.formatMessage(messages.video)} />
      </div>
    );
  }

  render () {
    const { card } = this.props;

    if (card === null) {
      return null;
    }

    switch(card.get('type')) {
    case 'link':
      return this.renderLink();
    case 'photo':
      return this.renderPhoto();
    case 'video':
      return this.renderVideo();
    case 'rich':
    default:
      return null;
    }
  }

}
