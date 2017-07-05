# frozen_string_literal: true

class ActivityPub::FollowSerializer < ActiveModel::Serializer
  attributes :type, :actor
  attribute :virtual_object, key: :object

  def serializable_hash(*_)
    result = super
    result.compact!
    result
  end

  def type
    'Follow'
  end

  def actor
    object.account && ActivityPub::TagManager.instance.uri_for(object.account)
  end

  def virtual_object
    ActivityPub::TagManager.instance.uri_for(object.target_account)
  end
end
