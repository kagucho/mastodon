# frozen_string_literal: true

class Settings::ExportsController < ApplicationController
  layout 'admin'

  before_action :authenticate_user!

  def show
    @export = Export.new(current_account)
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
  end
end
