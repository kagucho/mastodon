# frozen_string_literal: true

require 'sidekiq'

module SidekiqBudget
  module_function

  def configure
    Sidekiq.configure_client do |config|
      config.client_middleware do |chain|
        chain.add ClientMiddleware
      end
    end

    Sidekiq.configure_server do |config|
      config.client_middleware do |chain|
        chain.add ClientMiddleware
      end

      config.server_middleware do |chain|
        chain.add ServerMiddleware
      end
    end
  end

  def with(specified)
    original = $SIDEKIQ_BUDGET
    $SIDEKIQ_BUDGET = specified
    yield
  ensure
    $SIDEKIQ_BUDGET = original
  end
end

require 'sidekiq_budget/client_middleware'
require 'sidekiq_budget/controller_concern'
require 'sidekiq_budget/exhausted'
require 'sidekiq_budget/server_middleware'
require 'sidekiq_budget/worker'
