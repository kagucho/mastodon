# frozen_string_literal: true

require 'sidekiq-scheduler'

class Scheduler::SubscriptionsCleanupScheduler
  include SidekiqBudget::Worker

  def perform
    Subscription.expired.in_batches.delete_all
  end
end
