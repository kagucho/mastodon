# frozen_string_literal: true

require 'rails_helper'

RSpec.describe PrecomputeFeedService do
  subject { PrecomputeFeedService.new }

  describe 'call' do
    let(:account) { Fabricate(:account) }
    it 'fills a user timeline with statuses' do
      account = Fabricate(:account)
      followed_account = Fabricate(:account)
      Fabricate(:follow, account: account, target_account: followed_account)
      status = Fabricate(:status, account: followed_account)

      subject.call(account)

      expect(Redis.current.zscore(FeedManager.instance.key(:home, account.id), status.id)).to eq status.id
    end
  end
end
