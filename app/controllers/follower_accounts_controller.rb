# frozen_string_literal: true

class FollowerAccountsController < ApplicationController
  include AccountControllerConcern

  def index
    @follows = Follow.where(target_account: @account).recent.page(params[:page]).per(FOLLOW_PER_PAGE).preload(:account)
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET} #{ContentSecurityPolicy.digest(@card_style)}"
  end
end
