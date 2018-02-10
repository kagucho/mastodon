# frozen_string_literal: true

class ActivityPub::ProcessingWorker
  include SidekiqBudget::Worker

  sidekiq_options backtrace: true

  def perform(account_id, body)
    ActivityPub::ProcessCollectionService.new.call(body, Account.find(account_id), override_timestamps: true)
  end
end
