# frozen_string_literal: true

class MergeWorker
  include SidekiqBudget::Worker

  sidekiq_options queue: 'pull'

  def perform(from_account_id, into_account_id)
    FeedManager.instance.merge_into_timeline(Account.find(from_account_id), Account.find(into_account_id))
  end
end
