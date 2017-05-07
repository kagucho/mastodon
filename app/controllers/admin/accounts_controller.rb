# frozen_string_literal: true

module Admin
  class AccountsController < BaseController
    before_action :set_account, only: [:show, :subscribe, :unsubscribe, :redownload]
    before_action :require_remote_account!, only: [:subscribe, :unsubscribe, :redownload]

    def index
      @accounts = filtered_accounts.page(params[:page])
      set_csp
    end

    def show
      set_csp
    end

    def subscribe
      Pubsubhubbub::SubscribeWorker.perform_async(@account.id)
      redirect_to admin_account_path(@account.id)
    end

    def unsubscribe
      UnsubscribeService.new.call(@account)
      redirect_to admin_account_path(@account.id)
    end

    def redownload
      @account.avatar = @account.avatar_remote_url
      @account.header = @account.header_remote_url
      @account.save!

      redirect_to admin_account_path(@account.id)
    end

    private

    def set_account
      @account = Account.find(params[:id])
    end

    def require_remote_account!
      redirect_to admin_account_path(@account.id) if @account.local?
    end

    def filtered_accounts
      AccountFilter.new(filter_params).results
    end

    def filter_params
      params.permit(
        :local,
        :remote,
        :by_domain,
        :silenced,
        :recent,
        :suspended,
        :username,
        :display_name,
        :email,
        :ip
      )
    end

    def set_csp
      response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
    end
  end
end
