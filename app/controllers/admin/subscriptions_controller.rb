# frozen_string_literal: true

module Admin
  class SubscriptionsController < BaseController
    def index
      @subscriptions = ordered_subscriptions.page(requested_page)
      response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
    end

    private

    def ordered_subscriptions
      Subscription.order(id: :desc).includes(:account)
    end

    def requested_page
      params[:page].to_i
    end
  end
end
