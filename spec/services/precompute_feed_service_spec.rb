# frozen_string_literal: true

require 'rails_helper'

RSpec.describe PrecomputeFeedService do
  subject { PrecomputeFeedService.new }

  describe 'call' do
    context 'when user is continuously active' do
      it 'fills with FeedManager::MAX_ITEMS statuses at most since the last update' do
        stub_const('FeedManager::MAX_ITEMS', 1)
        last_sign_in_at = (User::FEED_UPDATED_DURATION + 1.day).ago
        user = Fabricate(:user, current_sign_in_at: last_sign_in_at, last_sign_in_at: last_sign_in_at)
        last_update = Fabricate(:status, account: user.account, created_at: last_sign_in_at + User::FEED_UPDATED_DURATION - 1.day)
        last = Fabricate(:status, account: user.account, created_at: last_sign_in_at + User::FEED_UPDATED_DURATION)
        first = Fabricate(:status, account: user.account, created_at: last_sign_in_at + User::FEED_UPDATED_DURATION + 1.day)

        subject.call(user.account)

        expect(Redis.current.zrange(FeedManager.instance.key(:home, user.account.id), 0, 1)).to eq [first.id.to_s]
      end
    end

    context 'when user is not continuously active' do
      it 'fills with FeedManager::MIN_ITEMS statuses at most in the last FeedManager::MIN_ID_RANGE' do
        stub_const('FeedManager::MIN_ITEMS', 1)
        last_sign_in_at = (User::FEED_UPDATED_DURATION + 1.day).ago
        user = Fabricate(:user, current_sign_in_at: nil)
        old = Fabricate(:status, id: 1, account: user.account)
        last = Fabricate(:status, id: 2, account: user.account)
        first = Fabricate(:status, id: FeedManager::MIN_ID_RANGE + 1, account: user.account)

        subject.call(user.account)

        expect(Redis.current.zrange(FeedManager.instance.key(:home, user.account.id), 0, 1)).to eq [first.id.to_s]
      end
    end

    it 'fills a user timeline with statuses' do
      user = Fabricate(:user)
      followed_account = Fabricate(:account)
      Fabricate(:follow, account: user.account, target_account: followed_account)
      reblog = Fabricate(:status, account: followed_account)
      status = Fabricate(:status, account: user.account, reblog: reblog)

      subject.call(user.account)

      expect(Redis.current.zscore(FeedManager.instance.key(:home, user.account.id), reblog.id)).to eq status.id
    end

    it 'does not raise an error even if it could not find any status' do
      user = Fabricate(:user)
      subject.call(user.account)
    end

    it 'filters statuses' do
      user = Fabricate(:user)
      muted_account = Fabricate(:account)
      Fabricate(:mute, account: user.account, target_account: muted_account)
      reblog = Fabricate(:status, account: muted_account)
      status = Fabricate(:status, account: user.account, reblog: reblog)

      subject.call(user.account)

      expect(Redis.current.zscore(FeedManager.instance.key(:home, user.account.id), reblog.id)).to eq nil
    end
  end
end
