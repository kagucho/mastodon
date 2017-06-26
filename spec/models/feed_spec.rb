require 'rails_helper'

RSpec.describe Feed, type: :model do
  describe '#get' do
    it 'gets statuses with ids in the range' do
      account = Fabricate(:account)
      Fabricate(:status, account: account, id: 1)
      Fabricate(:status, account: account, id: 2)
      Fabricate(:status, account: account, id: 3)
      Fabricate(:status, account: account, id: 10)
      Redis.current.zadd(FeedManager.instance.key(:home, account.id),
                        [[4, 'deleted'], [3, 'val3'], [2, 'val2'], [1, 'val1']])

      feed = Feed.new(:home, account)
      results = feed.get(3, nil, 0)

      expect(results.map(&:id)).to eq [3, 2, 1]
      expect(results.first.attributes.keys).to eq %w(id updated_at)
    end

    context 'when regeneration flag exists' do
      let(:last_sign_in_at) { Time.now }
      let(:user) { Fabricate(:user, current_sign_in_at: last_sign_in_at, last_sign_in_at: last_sign_in_at) }

      before { Redis.current.set("account:#{user.account.id}:regeneration", 1) }

      it 'gets from database when Redis gives no statuses' do
        old = Fabricate(:status, account: user.account, created_at: last_sign_in_at - 1.day)
        last_updated = Fabricate(:status, account: user.account, created_at: last_sign_in_at)
        new = Fabricate(:status, account: user.account, created_at: last_sign_in_at + User::FEED_UPDATED_DURATION)

        feed = Feed.new(:home, user.account)
        results = feed.get(3, nil, 0)

        expect(results.pluck(:id)).to eq [new.id, old.id]
      end

      it 'complements with database when it has new statuses Redis does not have' do
        in_redis = Fabricate(:status, account: user.account)
        new = Fabricate(:status, account: user.account)
        Redis.current.zadd(FeedManager.instance.key(:home, user.account.id), in_redis.id, in_redis.id)

        feed = Feed.new(:home, user.account)
        results = feed.get(1, nil, 0)

        expect(results.pluck(:id)).to eq [new.id]
      end

      it 'complements with database when Redis could not give sufficient results' do
        old = Fabricate(:status, account: user.account)
        in_redis = Fabricate(:status, account: user.account)
        new = Fabricate(:status, account: user.account)
        Redis.current.zadd(FeedManager.instance.key(:home, user.account.id), in_redis.id, in_redis.id)

        feed = Feed.new(:home, user.account)
        results = feed.get(3, nil, 0)

        expect(results.pluck(:id)).to eq [new.id, in_redis.id, old.id]
      end
    end

    context 'when regeneration flag does not exists' do
      it 'fall backs to database if Redis could not fill feed' do
        account = Fabricate(:account)
        statuses = 2.times.map { Fabricate(:status, account: account) }
        Redis.current.zadd(FeedManager.instance.key(:home, account.id), statuses[1].id, statuses[1].id)

        feed = Feed.new(:home, account)
        results = feed.get(2, nil, 0)

        expect(results.pluck(:id)).to eq statuses.pluck(:id).reverse
      end
    end
  end
end
