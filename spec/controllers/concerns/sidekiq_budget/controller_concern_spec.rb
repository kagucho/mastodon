# frozen_string_literal: true

require 'rails_helper'

describe ActionController::Base, type: :controller do
  controller do
    include SidekiqBudget::ControllerConcern

    limit_sidekiq_budget

    def index
      head 200
    end
  end

  describe 'limit_sidekiq_budget' do
    it 'limits Sidekiq budget for client' do
      expect(SidekiqBudget).to receive(:with) do |key, &block|
        expect(key).to eq '192.0.2.1'
        block.call
      end

      controller.request.remote_addr = '192.0.2.1'
      get :index
    end
  end
end
