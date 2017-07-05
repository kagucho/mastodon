# frozen_string_literal: true

class ActivityPub::FollowIntentSerializer < ActiveModel::Serializer
  attributes :type, :summary

  has_one :target, serializer: ActivityPub::FollowSerializer do
    Follow.new(target_account: object)
  end

  def type
    'https://akihikodaki.github.io/activity-intent/ns#Intent'
  end

  def summary
    "Following @#{object.username}"
  end
end
