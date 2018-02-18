# frozen_string_literal: true
require 'sidekiq-scheduler'

class Scheduler::DeadRecordsCleanupScheduler
  include Sidekiq::Worker

  DATA_RETENTION = 30.days

  def perform
    Account.where(halted: false, suspended: true).where('updated_at < ?', DATA_RETENTION.ago).reorder(nil).find_in_batches do |accounts|
      accounts.each { |account| HaltAccountService.new.call account }
    end
  end
end
