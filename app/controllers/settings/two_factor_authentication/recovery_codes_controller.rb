# frozen_string_literal: true

module Settings
  module TwoFactorAuthentication
    class RecoveryCodesController < ApplicationController
      layout 'admin'

      before_action :authenticate_user!

      def create
        @recovery_codes = current_user.generate_otp_backup_codes!
        current_user.save!
        flash[:notice] = I18n.t('two_factor_authentication.recovery_codes_regenerated')
        response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
        render :index
      end
    end
  end
end
