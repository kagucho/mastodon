# frozen_string_literal: true

class ContentSecurityPolicy
  asset_host = Rails.configuration.action_controller.asset_host

  if Rails.env.development?
    ASSET = asset_host || "'self' http://localhost:8080"
    ASSET_VIEW = asset_host ? asset_host + " 'self' http://localhost:8080" : "'self' http://localhost:8080"
  else
    ASSET = asset_host || "'self'"
    ASSET_VIEW = asset_host ? asset_host + " 'self'" : "'self'"
  end

  VIEW = "'self'"

  def self.digest(string)
    "'sha256-#{Base64.strict_encode64(OpenSSL::Digest::SHA256.digest(string || ''))}'"
  end
end
