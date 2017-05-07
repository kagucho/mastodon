# frozen_string_literal: true

module Settings
  module TwoFactorAuthentication
    class ConfirmationsController < ApplicationController
      layout 'admin'

      before_action :authenticate_user!

      def new
        prepare_two_factor_form
      end

      def create
        if current_user.validate_and_consume_otp!(confirmation_params[:code])
          flash[:notice] = I18n.t('two_factor_authentication.enabled_success')

          current_user.otp_required_for_login = true
          @recovery_codes = current_user.generate_otp_backup_codes!
          current_user.save!

          response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
          render 'settings/two_factor_authentication/recovery_codes/index'
        else
          flash.now[:alert] = I18n.t('two_factor_authentication.wrong_code')
          prepare_two_factor_form
          render :new
        end
      end

      private

      def confirmation_params
        params.require(:form_two_factor_confirmation).permit(:code)
      end

      def prepare_two_factor_form
        @confirmation = Form::TwoFactorConfirmation.new
        @provision_url = current_user.otp_provisioning_uri(current_user.email, issuer: Rails.configuration.x.local_domain)
        response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
        @qrcode = RQRCode::QRCode.new(@provision_url)
      end
    end
  end
end
