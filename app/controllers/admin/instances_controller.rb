# frozen_string_literal: true

module Admin
  class InstancesController < BaseController
    def index
      @instances = ordered_instances
      response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
    end

    private

    def paginated_instances
      Account.remote.by_domain_accounts.page(params[:page])
    end
    helper_method :paginated_instances

    def ordered_instances
      paginated_instances.map { |account| Instance.new(account) }
    end
  end
end
