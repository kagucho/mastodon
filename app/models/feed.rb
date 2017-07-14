# frozen_string_literal: true

class Feed
  def initialize(type, account)
    @type    = type
    @account = account
  end

  # Make sure to pass a reasonable value for since_id. A query without since_id
  # could be heavy if the specified number of statuses is not available. In the
  # case, it retrieves ALL statuses before max_id.
  def get(limit, max_id, since_id)
    if redis.exists("account:#{@account.id}:regeneration")
      if @account.user.feed_persistent?
        last_updated_id = @account.user.last_updated_feed_status_id

        # This can retrieve statuses with id <= last_updated_id.
        statuses_from_redis = from_redis(limit, max_id, since_id)

        # This can retrieve statuses with id > last_updated_id.
        statuses = from_database(limit, max_id, statuses_from_redis.first&.id || last_updated_id || since_id)

        statuses += statuses_from_redis
        statuses = statuses[0, limit]

        # This can retrieve statuses with id < last_updated_id.
        statuses += from_database(limit - statuses.size,
                                  statuses_from_redis.last&.id || max_id || last_updated_id,
                                  since_id)

        # Sum of the above three queries.
        statuses
      else
        from_database(limit, max_id, since_id)
      end
    else
      statuses = from_redis(limit, max_id, since_id)

      statuses += from_database(limit - statuses.size,
                                statuses.last&.id || max_id,
                                since_id)

      statuses
    end
  end

  private

  def from_redis(limit, max_id, since_id)
    max_id     = '+inf' if max_id.blank?
    unhydrated = redis.zrevrangebyscore(key, "(#{max_id}", "(#{since_id}", limit: [0, limit], with_scores: true).map(&:last).map(&:to_i)
    Status.where(id: unhydrated).cache_ids
  end

  def from_database(limit, max_id, since_id)
    Status.as_home_timeline(@account)
          .paginate_by_max_id(limit, max_id, since_id)
          .reject { |status| FeedManager.instance.filter?(:home, status, @account.id) }
  end

  def key
    FeedManager.instance.key(@type, @account.id)
  end

  def redis
    Redis.current
  end
end
