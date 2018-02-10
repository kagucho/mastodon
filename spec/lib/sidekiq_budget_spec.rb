# frozen_string_literal: true

require 'sidekiq_budget'
require 'sidekiq/testing'
require 'spec_helper'

describe SidekiqBudget do
  describe 'configure' do
    before do
      client_chain = double
      configure = Sidekiq.method(:configure_server)

      expect(client_chain).to receive(:add).with(SidekiqBudget::ClientMiddleware)

      expect(Sidekiq).to receive(:configure_server) do |&config_block|
        config = double

        expect(config).to receive(:client_middleware).and_yield(client_chain)
        expect(config).to receive(:server_middleware) do |&block|
          Sidekiq::Testing.server_middleware &block
        end

        config_block.call config
      end

      allow_any_instance_of(Sidekiq::ProcessSet).to receive(:size).and_return(1)

      SidekiqBudget.configure
    end

    class SpecWorker
      include Sidekiq::Worker

      def perform
      end
    end

    context 'with budget' do
      before { $SIDEKIQ_BUDGET = 'budget' }

      it 'lets worker raise SidekiqBudget::Exhausted if the operation has taken more than 64 seconds' do
        Sidekiq::Testing.inline! do
          allow(Benchmark).to receive(:realtime).and_return(64)
          SpecWorker.perform_async
          expect{ SpecWorker.perform_async }.to raise_error SidekiqBudget::Exhausted
        end
      end

      it 'takes more time to exhaust budget with more processors' do
        allow_any_instance_of(Sidekiq::ProcessSet).to receive(:size).and_return(2)

        Sidekiq::Testing.inline! do
          allow(Benchmark).to receive(:realtime).and_return(64)
          SpecWorker.perform_async
          SpecWorker.perform_async
          expect{ SpecWorker.perform_async }.to raise_error SidekiqBudget::Exhausted
        end
      end

      it 'does not let worker raise if the operation has not taken more than 64 seconds' do
        Sidekiq::Testing.inline! do
          allow(Benchmark).to receive(:realtime).and_return(63)
          SpecWorker.perform_async
          expect{ SpecWorker.perform_async }.not_to raise_error
        end
      end

      it 'does not let worker raise when the operation starts' do
        Sidekiq::Testing.inline! do
          expect{ SpecWorker.perform_async }.not_to raise_error
        end
      end

      it 'lets worker inherit $SIDEKIQ_BUDGET' do
        expect_any_instance_of(SpecWorker).to receive(:perform) do
          expect($SIDEKIQ_BUDGET).to eq 'budget'
        end

        Sidekiq::Testing.fake! do
          SpecWorker.perform_async
          $SIDEKIQ_BUDGET = nil
          SpecWorker.drain
        end
      end
    end

    context 'without budget' do
      before { $SIDEKIQ_BUDGET = nil }

      it 'does not raise' do
        expect{ SpecWorker.perform_async }.not_to raise_error
      end
    end
  end

  describe 'with' do
    it 'executes the given block with specified budget' do
      budget = nil

      SidekiqBudget.with('specified_budget') { budget = $SIDEKIQ_BUDGET }

      expect(budget).to eq 'specified_budget'
    end

    it 'restores the budget' do
      $SIDEKIQ_BUDGET = 'original_budget'
      SidekiqBudget.with('specified_budget') { }
      expect($SIDEKIQ_BUDGET).to eq 'original_budget'
    end
  end
end
