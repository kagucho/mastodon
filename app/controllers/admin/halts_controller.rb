# frozen_string_literal: true

module Admin
  class HaltsController < BaseController
    before_action :set_account

    def create
      authorize @account, :halt?
      Admin::HaltWorker.perform_async(@account.id)
      log_action :halt, @account
      redirect_to admin_accounts_path
    end

    def destroy
      authorize @account, :restore?
      @account.restore!
      log_action :restore, @account
      redirect_to admin_accounts_path
    end

    private

    def set_account
      @account = Account.find(params[:account_id])
    end
  end
end
