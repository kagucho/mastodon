# frozen_string_literal: true
# == Schema Information
#
# Table name: account_relations
#
#  account_id        :integer          not null
#  target_account_id :integer          not null
#  created_at        :datetime         not null
#  updated_at        :datetime         not null
#  type              :enum             not null
#

class AccountRelation < ApplicationRecord
  include Paginable

  belongs_to :account, required: true
  belongs_to :target_account, class_name: 'Account', required: true

  validates :account_id, uniqueness: { scope: :target_account_id }

  after_create  :remove_account_relation_cache
  after_destroy :remove_account_relation_cache

  enum type: { block: 'block', mute: 'mute' }

  private

  def remove_account_relation_cache
    Rails.cache.delete("exclude_account_ids_for:#{account_id}")
  end
end
