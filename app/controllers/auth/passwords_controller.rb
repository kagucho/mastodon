# frozen_string_literal: true

class Auth::PasswordsController < Devise::PasswordsController
  layout 'auth'

  def new
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
    super
  end
end
