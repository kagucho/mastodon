# frozen_string_literal: true

class Admin::HaltWorker
  include Sidekiq::Worker

  sidekiq_options queue: 'pull'

  def perform(account_id, remove_user = false)
    HaltAccountService.new.call(Account.find(account_id), remove_user: remove_user)
  end
end
