class CountCache
  def initialize(klass, name, column, &test)
    on_destroy_method = method(:on_destroy)
    on_update_method = method(:on_update)
    redis_key_prefix = "count_cache:#{klass.table_name}:#{name}:"

    @redis_count_key = redis_key_prefix + 'count'
    @redis_id_key = redis_key_prefix + 'id'
    @redis_time_key = redis_key_prefix + 'time'
    @scope = klass.method(name)

    klass.after_commit on: :destroy do
      on_destroy_method.call id, test.call(send(column))
    end

    klass.after_commit on: :update do
      change = previous_changes[column]
      next if change.nil?

      on_update_method.call id, change.map(&test)
    end
  end

  def fetch
    count = nil
    time = Redis.current.get(@redis_time_key)

    if time.nil?
      relation = @scope.call
      count = relation.count
      first = relation.first

      if first.present?
        Redis.current.mset @redis_count_key, count, @redis_id_key, first.id, @redis_time_key, Time.new.to_i
      end
    elsif Time.at(time.to_i) < 10.minutes.ago
      Redis.current.watch @redis_id_key do
        count = Redis.current.get(@redis_count_key).to_i
        id = Redis.current.get(@redis_id_key).to_i

        relation = @scope.call.where('id > ?', id)
        count += relation.count
        first = relation.first

        if first.present?
          Redis.current.multi do |multi|
            multi.mset @redi_count_key, count, @redis_id_key, first.id, @redis_time_key, Time.new.to_i
          end
        end
      end
    else
      count = Redis.current.get(@redis_count_key).to_i
    end

    count
  end

  private

  def on_destroy(id, matches)
    return unless matches

    last_id_s = Redis.current.get(@redis_id_key)
    return if last_id_s.nil? || last_id_s.to_i < id

    Redis.current.decr @redis_count_key
  end

  def on_update(id, matches)
    return if matches[0] == matches[1]

    last_id_s = Redis.current.get(@redis_id_key)
    return if last_id_s.nil? || last_id_s.to_i < id

    if matches[1]
      Redis.current.incr @redis_count_key
    else
      Redis.current.decr @redis_count_key
    end
  end
end
