# frozen_string_literal: true

module UserTrackingConcern
  extend ActiveSupport::Concern

  included do
    before_action :set_user_activity, if: %i(user_signed_in? user_needs_sign_in_update?)
  end

  private

  def set_user_activity
    # Mark as signed-in today
    current_user.update_tracked_fields!(request)

    # Regenerate feed if needed
    regenerate_feed! if user_needs_feed_update?
  end

  def user_needs_sign_in_update?
    current_user.current_sign_in_at.nil? || current_user.current_sign_in_at < User::UPDATE_SIGN_IN_DURATION.ago
  end

  def user_needs_feed_update?
    current_user.last_sign_in_at < User::FEED_UPDATED_DURATION.ago
  end

  def regenerate_feed!
    Redis.current.setnx("account:#{current_user.account_id}:regeneration", true) == 1 && Redis.current.expire("account:#{current_user.account_id}:regeneration", 3_600 * 24)
    RegenerationWorker.perform_async(current_user.account_id)
  end
end
