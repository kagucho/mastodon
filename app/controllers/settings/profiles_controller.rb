# frozen_string_literal: true

class Settings::ProfilesController < ApplicationController
  include ObfuscateFilename

  layout 'admin'

  before_action :authenticate_user!
  before_action :set_account

  obfuscate_filename [:account, :avatar]
  obfuscate_filename [:account, :header]

  def show
    set_csp
  end

  def update
    if @account.update(account_params)
      redirect_to settings_profile_path, notice: I18n.t('generic.changes_saved_msg')
    else
      set_csp
      render :show
    end
  end

  private

  def account_params
    params.require(:account).permit(:display_name, :note, :avatar, :header, :locked)
  end

  def set_account
    @account = current_user.account
  end

  def set_csp
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
  end
end
