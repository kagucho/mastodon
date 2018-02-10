# frozen_string_literal: true

module SidekiqBudget::Worker
  module_function

  def included(klass)
    klass.include Sidekiq::Worker
  end

  def perform_in(*)
    SidekiqBudget.with(nil) { super }
  end

  def perform_at(*)
    SidekiqBudget.with(nil) { super }
  end

  private
end
