# frozen_string_literal: true

require 'rails_helper'

describe ProcessActivityController, type: :controller do
  describe 'show' do
    VALID_FOLLOW_INTENT = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "https://akihikodaki.github.io/ns#Intent",
  "object": {
    "type": "Follow",
    "object": {
      "id": "http://instance/path/to/person",
      "type": "Person"
    }
  },
  "summary": "Following Person"
}
JSON

    VALID_CREATE_NOTE_INTENT = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "https://akihikodaki.github.io/ns#Intent",
  "object": {
    "type": "Create",
    "audience": "https://mastodon.social/ns/activity-mastodon#public",
    "object": {
      "type": "Note",
      "content": "Content of a note",
      "mediaType": "text/plain"
    }
  },
  "summary": "Creating a note"
}
JSON

    VALID_NOTE_INTENT = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "https://akihikodaki.github.io/ns#Intent",
  "object": {
    "type": "Note",
    "audience": "https://mastodon.social/ns/activity-mastodon#public",
    "content": "Content of a note",
    "mediaType": "text/plain"
  },
  "summary": "Creating a note"
}
JSON

    NOTE_INTENT_WITHOUT_OBJECT_MEDIA_TYPE = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "https://akihikodaki.github.io/ns#Intent",
  "object": {
    "type": "Note",
    "audience": "https://mastodon.social/ns/activity-mastodon#public",
    "content": "Content of a note"
  },
  "summary": "Creating a note"
}
JSON

    NOTE_INTENT_WITH_UNKNOWN_OBJECT_MEDIA_TYPE = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "https://akihikodaki.github.io/ns#Intent",
  "object": {
    "type": "Note",
    "audience": "https://mastodon.social/ns/activity-mastodon#public",
    "content": "Content of a note",
    "mediaType": "text/html"
  },
  "summary": "Creating a note"
}
JSON

    UNKNOWN_TYPE = <<JSON
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "Object",
  "summary": "Unknown object"
}
JSON

    INVALID_JSON = '{'

    # ActivityPub Extension for Intent
    # https://akihikodaki.github.io/activity-intent/activitypub.html
    context 'for intents' do
      # ActivityPub
      # 6.4 Follow Activity
      # https://www.w3.org/TR/activitypub/#follow-activity-outbox
      context 'for follow activity' do
        it 'redirects to authorize_follow' do
          stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_FOLLOW_INTENT)
          get :show, params: { url: 'https://instance/path/to/activity' }
          expect(response).to redirect_to 'http://test.host/authorize_follow?acct=http%3A%2F%2Finstance%2Fpath%2Fto%2Fperson'
        end
      end

      context 'for create activity' do
        # ActivityPub
        # 6.1 Create Activity
        # https://www.w3.org/TR/activitypub/#create-activity-outbox
        context 'with wrapping' do
          it 'performs the activity' do
            stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_CREATE_NOTE_INTENT)
            get :show, params: { url: 'https://instance/path/to/activity' }
            expect(response).to redirect_to 'http://test.host/intents/statuses#status=Content%2520of%2520a%2520note&visibility=public'
          end
        end

        # ActivityPub
        # 6.1.1 Object creation without a Create Activity
        # https://www.w3.org/TR/activitypub/#object-without-create
        context 'without wrapping' do
          it 'performs the activity' do
            stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_NOTE_INTENT)
            get :show, params: { url: 'https://instance/path/to/activity' }
            expect(response).to redirect_to 'http://test.host/intents/statuses#status=Content%2520of%2520a%2520note&visibility=public'
          end
        end

        context 'for note' do
          it 'redirects to intents/statuses' do
            stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_NOTE_INTENT)
            get :show, params: { url: 'https://instance/path/to/activity' }
            expect(response).to redirect_to 'http://test.host/intents/statuses#status=Content%2520of%2520a%2520note&visibility=public'
          end

          # Activity Vocabulary
          # https://www.w3.org/TR/activitystreams-vocabulary/#dfn-content
          # > By default, the value of content is HTML.
          it 'returns 422 without mediaType' do
            stub_request(:get, 'https://instance/path/to/activity').to_return(body: NOTE_INTENT_WITHOUT_OBJECT_MEDIA_TYPE)
            get :show, params: { url: 'https://instance/path/to/activity' }
            expect(response).to have_http_status 422
          end

          it 'returns 422 for values other than text/plain in mediaType property' do
            stub_request(:get, 'https://instance/path/to/activity').to_return(body: NOTE_INTENT_WITH_UNKNOWN_OBJECT_MEDIA_TYPE)
            get :show, params: { url: 'https://instance/path/to/activity' }
            expect(response).to have_http_status 422
          end
        end
      end
    end

    # URL Scheme for Activity Streams 2.0
    # https://akihikodaki.github.io/activity-intent/scheme
    # > 3. web+activity+https URL scheme
    it 'understands web+activity+https scheme' do
      stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_FOLLOW_INTENT)
      get :show, params: { url: 'web+activity+https://instance/path/to/activity' }
      expect(response).to redirect_to 'http://test.host/authorize_follow?acct=http%3A%2F%2Finstance%2Fpath%2Fto%2Fperson'
    end

    # URL Scheme for Activity Streams 2.0
    # https://akihikodaki.github.io/activity-intent/scheme
    # > If a consumer of URLs understands web+activity+https scheme, it SHOULD
    # > understand https scheme for the same purposes to achieve better
    # > interoperability.
    it 'understands https scheme' do
      stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_FOLLOW_INTENT)
      get :show, params: { url: 'https://instance/path/to/activity' }
      expect(response).to redirect_to 'http://test.host/authorize_follow?acct=http%3A%2F%2Finstance%2Fpath%2Fto%2Fperson'
    end

    # URL Scheme for Activity Streams 2.0
    # https://akihikodaki.github.io/activity-intent/scheme
    # > The user agent MUST set application/ld+json; profile="https://www.w3.org/ns/activitystreams"
    # > for the Accept header field defined in [RFC7231], Section 5.3.2.
    #
    # ActivityPub
    # 3.2 Retreiving objects
    # https://www.w3.org/TR/activitypub/#retrieving-objects
    # > The HTTP GET method may be dereferenced against an object's id property
    # > to retrieve the activity.
    #
    # > The client must specify an Accept header with the
    # > application/ld+json; profile="https://www.w3.org/ns/activitystreams"
    # > media type in order to retrieve the activity.
    it "uses HTTP GET method with Accept header" do
      stub_request(:get, 'https://instance/path/to/activity').to_return(body: VALID_FOLLOW_INTENT)
      get :show, params: { url: 'https://instance/path/to/activity' }
      expect(a_request(:get, 'https://instance/path/to/activity')
               .with(headers: { 'Accept' => 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"' }))
        .to have_been_made
    end

    it 'returns 400 for invalid URL' do
      get :show, params: { url: ':' }
      expect(response).to have_http_status 400
    end

    it 'returns 422 for HTTP failure' do
      stub_request(:get, 'https://instance/path/to/activity').to_timeout
      get :show, params: { url: 'https://instance/path/to/activity' }
      expect(response).to have_http_status 422
    end

    it 'returns 422 for invalid JSON' do
      stub_request(:get, 'https://instance/path/to/activity').to_return(body: INVALID_JSON)
      get :show, params: { url: 'https://instance/path/to/activity' }
      expect(response).to have_http_status 422
    end

    it 'returns 422 for unknown type' do
      stub_request(:get, 'https://instance/path/to/activity').to_return(body: UNKNOWN_TYPE)
      get :show, params: { url: 'https://instance/path/to/activity' }
      expect(response).to have_http_status 422
    end
  end
end
