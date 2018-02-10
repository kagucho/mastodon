# frozen_string_literal: true

require 'doorkeeper/grape/authorization_decorator'

class Rack::Attack
  class Request
    def authenticated_token
      return @token if defined?(@token)

      @token = Doorkeeper::OAuth::Token.authenticate(
        Doorkeeper::Grape::AuthorizationDecorator.new(self),
        *Doorkeeper.configuration.access_token_methods
      )
    end

    def authenticated_user_id
      authenticated_token&.resource_owner_id
    end

    def unauthenticated?
      !authenticated_user_id
    end

    def api_request?
      path.start_with?('/api')
    end

    def web_request?
      !api_request?
    end
  end

  CALLBACK_PATHS = [
    # /api/inbox, /api/:account_id:/inbox, and /api/salmon has lots of requests
    # and should not be comprehensively throttled.

    # fetches remote accounts
    %r{/api/accounts/search},

    # sends activities to remote accounts
    %r{/api/account/.*?/follow},
    %r{/api/account/.*?/unfollow},
    %r{/api/account/.*?/block},
    %r{/api/account/.*?/unblock},

    # registers Pubsubhubbub callbacks
    %r{/api/push},

    # sends activities to remote accounts
    %r{/api/v1/follow},
    %r{/api/v1/follow_request/authorize},
    %r{/api/v1/follow_request/reject},

    # fetches remote accounts and statuses
    %r{/api/v1/search},

    # sends activities to remote accounts
    %r{/api/v1/status/.*?/reblog},
    %r{/api/v1/status/.*?/unreblog},
    %r{/api/v1/status/.*?/favourite},
    %r{/api/v1/status/.*?/unfavourite},

    # fetches remote accounts
    %r{/authorize_follow},
  ].freeze

  PROTECTED_PATHS = %w(
    /auth/sign_in
    /auth
    /auth/password
  ).freeze

  CALLBACK_PATHS_REGEX = Regexp.union(CALLBACK_PATHS.map { |path| /\A#{Regexp.escape(path)}/ })
  PROTECTED_PATHS_REGEX = Regexp.union(PROTECTED_PATHS.map { |path| /\A#{Regexp.escape(path)}/ })

  # Always allow requests from localhost
  # (blocklist & throttles are skipped)
  Rack::Attack.safelist('allow from localhost') do |req|
    # Requests are allowed if the return value is truthy
    '127.0.0.1' == req.ip || '::1' == req.ip
  end

  throttle('throttle_callback', limit: 300, period: 5.minutes) do |req|
    req.path =~ CALLBACK_PATHS_REGEX
  end

  throttle('throttle_authenticated_api', limit: 300, period: 5.minutes) do |req|
    req.api_request? && req.authenticated_user_id
  end

  throttle('throttle_unauthenticated_api', limit: 7_500, period: 5.minutes) do |req|
    req.ip if req.api_request?
  end

  throttle('protected_paths', limit: 25, period: 5.minutes) do |req|
    req.ip if req.post? && req.path =~ PROTECTED_PATHS_REGEX
  end

  self.throttled_response = lambda do |env|
    now        = Time.now.utc
    match_data = env['rack.attack.match_data']

    headers = {
      'Content-Type'          => 'application/json',
      'X-RateLimit-Limit'     => match_data[:limit].to_s,
      'X-RateLimit-Remaining' => '0',
      'X-RateLimit-Reset'     => (now + (match_data[:period] - now.to_i % match_data[:period])).iso8601(6),
    }

    [429, headers, [{ error: I18n.t('errors.429') }.to_json]]
  end
end
