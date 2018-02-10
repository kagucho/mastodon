# frozen_string_literal: true

class RemovalWorker
  include SidekiqBudget::Worker

  def perform(status_id)
    RemoveStatusService.new.call(Status.find(status_id))
  rescue ActiveRecord::RecordNotFound
    true
  end
end
