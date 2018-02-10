# frozen_string_literal: true

class ActivityPub::PostUpgradeWorker
  include SidekiqBudget::Worker

  sidekiq_options queue: 'pull'

  def perform(domain)
    Account.where(domain: domain)
           .where(protocol: :ostatus)
           .where.not(last_webfingered_at: nil)
           .in_batches
           .update_all(last_webfingered_at: nil)
  end
end
