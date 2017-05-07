# frozen_string_literal: true

class HomeController < ApplicationController
  before_action :authenticate_user!

  def index
    @body_classes           = 'app-body'
    @img_nonce              = SecureRandom.base64
    @style_nonce            = SecureRandom.base64
    @token                  = current_session.token
    @web_settings           = Web::Setting.find_by(user: current_user)&.data || {}
    @admin                  = Account.find_local(Setting.site_contact_username)
    @streaming_api_base_url = Rails.configuration.x.streaming_api_base_url

    response.headers['Content-Security-Policy'] = "child-src #{ContentSecurityPolicy::VIEW}; connect-src #{@streaming_api_base_url} #{ContentSecurityPolicy::VIEW}; default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; frame-src #{ContentSecurityPolicy::VIEW}; img-src #{ContentSecurityPolicy::ASSET_VIEW} 'nonce-#{@img_nonce}' data:; media-src #{ContentSecurityPolicy::ASSET_VIEW}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET} 'nonce-#{@style_nonce}'"
  end

  private

  def authenticate_user!
    redirect_to(single_user_mode? ? account_path(Account.first) : about_path) unless user_signed_in?
  end
end
