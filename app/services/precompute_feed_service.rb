# frozen_string_literal: true

class PrecomputeFeedService < BaseService
  def call(account)
    @account = account
    populate_feed
  end

  private

  attr_reader :account

  def populate_feed
    pairs = statuses.reverse_each.lazy.reject(&method(:status_filtered?)).map(&method(:process_status)).to_a

    redis.pipelined do
      redis.zadd(account_home_key, pairs) if pairs.any?
      redis.del("account:#{@account.id}:regeneration")
    end

    FeedManager.instance.trim(:home, account.id) if account.user.continuously_active?
  end

  def process_status(status)
    [status.id, status.reblog? ? status.reblog_of_id : status.id]
  end

  def status_filtered?(status)
    FeedManager.instance.filter?(:home, status, account.id)
  end

  def account_home_key
    FeedManager.instance.key(:home, account.id)
  end

  def statuses
    limit = nil
    since_id = nil

    if account.user.continuously_active?
      # Limit with MAX_ITEMS to ensure the old feed and the new feed becomes
      # continuous.
      limit = FeedManager::MAX_ITEMS
      since_id = account.user.last_updated_feed_status_id
    else
      last = Status.last
      return [] if last.nil?

      limit = FeedManager::MIN_ITEMS
      since_id = last.id - FeedManager::MIN_ID_RANGE
    end

    Status.as_home_timeline(account).order(account_id: :desc).paginate_by_max_id(limit, nil, since_id)
  end

  def redis
    Redis.current
  end
end
