# frozen_string_literal: true

class SidekiqBudget::ClientMiddleware
  def call(worker, job, queue, redis)
    job['budget'] = $SIDEKIQ_BUDGET
    yield
  end
end
