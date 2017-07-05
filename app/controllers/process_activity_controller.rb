# frozen_string_literal: true

class ProcessActivityController < ApplicationController
  IRI_ACTIVITY_MASTODON = 'https://mastodon.social/ns/activity-mastodon#'
  IRI_INTENT = 'https://akihikodaki.github.io/ns#Intent'

  def show
    url = /^(?:web\+activity\+)?(.*)/.match(params.require(:url))[1]

    begin
      request = Request.new(:get, url)
      request.add_headers(accept: 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"')
      response = request.perform
    rescue Addressable::URI::InvalidURIError
      head 400
      return
    rescue HTTP::Error
      raise Mastodon::ValidationError
    end

    begin
      activity = Oj.load(response)
    rescue Oj::Error
      raise Mastodon::ValidationError
    end

    raise Mastodon::ValidationError if activity.nil?

    if activity['type'] == IRI_INTENT
      object = activity['object']

      if object['type'] == 'Follow'
        follow_object = object['object']

        redirect_to authorize_follow_url(acct: follow_object['id'])
        return
      else
        create_object = object['type'] == 'Create' ? object['object'] : object

        if create_object['type'] == 'Note'
          values = {}

          audience = object['audience']
          unless audience.nil?
            raise Mastodon::ValidationError unless audience.start_with? IRI_ACTIVITY_MASTODON
            values['visibility'] = audience[IRI_ACTIVITY_MASTODON.size..-1]
          end

          content = create_object['content']
          values['status'] = content unless content.nil?

          media_type = create_object['mediaType']
          raise Mastodon::ValidationError if media_type&.casecmp('text/plain') != 0

          redirect_to intents_statuses_url(action: :new, anchor: Addressable::URI.new(query_values: values).query)
          return
        end
      end
    end

    unprocessable_entity
  end
end
