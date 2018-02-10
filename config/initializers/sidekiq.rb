# frozen_string_literal: true

require 'sidekiq_budget'

namespace = ENV.fetch('REDIS_NAMESPACE') { nil }
redis_params = { url: ENV['REDIS_URL'] }

if namespace
  redis_params [:namespace] = namespace
end

Sidekiq.configure_server do |config|
  config.redis = redis_params
end

Sidekiq.configure_client do |config|
  config.redis = redis_params
end

SidekiqBudget.configure
