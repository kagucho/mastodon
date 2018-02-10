# frozen_string_literal: true

module SidekiqBudget::ControllerConcern
  extend ActiveSupport::Concern

  included do
    def self.limit_sidekiq_budget(*args)
      around_action :sidekiq_budget, *args
    end

    private_class_method :limit_sidekiq_budget
  end

  private

  def sidekiq_budget(&block)
    SidekiqBudget.with request.remote_ip, &block
  end
end
