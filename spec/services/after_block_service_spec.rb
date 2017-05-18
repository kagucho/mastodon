require 'rails_helper'

RSpec.describe AfterBlockService do
  subject do
    -> { described_class.new.call(account, target_account) }
  end

  let(:account) { Fabricate(:account) }
  let(:target_account) { Fabricate(:account) }

  describe 'home timeline' do
    let(:status) { Fabricate(:status, account: target_account) }
    let(:other_account_status) { Fabricate(:status) }
    let(:home_timeline_key) { FeedManager.instance.key(:home, account.id) }

    before do
      Redis.current.del(home_timeline_key)
    end

    it "clears account's statuses" do
      Redis.current.set("subscribed:timeline:#{account.id}", '1')

      FeedManager.instance.push_bulk(:home, [account], status)
      FeedManager.instance.push_bulk(:home, [account], other_account_status)

      is_expected.to change {
        Redis.current.zrange(home_timeline_key, 0, -1)
      }.from([status.id.to_s, other_account_status.id.to_s]).to([other_account_status.id.to_s])
    end
  end
end
