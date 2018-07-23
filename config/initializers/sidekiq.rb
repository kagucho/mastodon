# frozen_string_literal: true

namespace    = ENV.fetch('REDIS_NAMESPACE') { nil }
redis_params = { url: ENV['REDIS_URL'] }

if namespace
  redis_params[:namespace] = namespace
end

$nekomanma_pp = false

class NekomanmaPpMiddleware
  def call(*args)
    pp *args if $nekomanma_pp
    yield
  end
end

Sidekiq.configure_server do |config|
  config.redis = redis_params

  config.server_middleware do |chain|
    chain.add SidekiqErrorHandler
  end
end

Sidekiq.configure_client do |config|
  config.redis = redis_params

  config.client_middleware do |chain|
    chain.add NekomanmaPpMiddleware
  end
end

Sidekiq::Logging.logger.level = ::Logger::const_get(ENV.fetch('RAILS_LOG_LEVEL', 'info').upcase.to_s)
