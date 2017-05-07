# frozen_string_literal: true

class Settings::ImportsController < ApplicationController
  layout 'admin'

  before_action :authenticate_user!
  before_action :set_account

  def show
    @import = Import.new
    set_csp
  end

  def create
    @import = Import.new(import_params)
    @import.account = @account

    if @import.save
      ImportWorker.perform_async(@import.id)
      redirect_to settings_import_path, notice: I18n.t('imports.success')
    else
      set_csp
      render :show
    end
  end

  private

  def set_account
    @account = current_user.account
  end

  def import_params
    params.require(:import).permit(:data, :type)
  end

  def set_csp
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
  end
end
