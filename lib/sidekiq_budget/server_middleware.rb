# frozen_string_literal: true

require 'benchmark'
require 'redis'
require 'sidekiq/api'

class SidekiqBudget::ServerMiddleware
  def call(worker, job, queue)
    budget = job['budget']

    if budget.nil?
      $SIDEKIQ_BUDGET = nil
      yield
    else
      rate = "sidekiq_budget:#{budget}"
      rate_value = Redis.current.get(rate)

      unless rate_value.nil? || rate_value.to_i < 64
        raise SidekiqBudget::Exhausted
      end

      time = Benchmark.realtime do
        $SIDEKIQ_BUDGET = budget
        yield
      end

      time /= Sidekiq::ProcessSet.new.size

      if time > 1
        Redis.current.multi do
          Redis.current.incrby rate, time.floor
          Redis.current.expire rate, 256
        end
      end
    end
  end
end
