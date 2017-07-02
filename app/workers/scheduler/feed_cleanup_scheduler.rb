# frozen_string_literal: true
require 'sidekiq-scheduler'

class Scheduler::FeedCleanupScheduler
  include Sidekiq::Worker

  def perform
    logger.info 'Cleaning out expired home feeds'

    redis.pipelined do
      users_with_expired_feed.pluck(:account_id).each do |account_id|
        redis.del(FeedManager.instance.key(:home, account_id))
      end
    end
  end

  private

  def users_with_expired_feed
    User.confirmed.feed_expired
  end

  def redis
    Redis.current
  end
end
