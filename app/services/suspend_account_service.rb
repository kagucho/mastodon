# frozen_string_literal: true

class SuspendAccountService < BaseService
  def call(account, **options)
    @account = account
    @options = options

    purge_user!
    purge_profile!
    purge_content!
  end

  private

  def purge_user!
    @account.user&.disable!
  end

  def purge_content!
    @account.statuses.reorder(nil).find_in_batches do |statuses|
      BatchedRemoveAccountStatusService.new.call(statuses)
    end

    NotificationWorker.push_bulk(@account.favourites) do |favourite|
      next if favourite.status.local?
      [build_unfavourite_xml(favourite), favourite.account_id, favourite.status.account_id]
    end
  end

  def purge_profile!
    @account.hidden = true
    @account.save!
  end

  def build_unfavourite_xml
    OStatus::AtomSerializer.render(OStatus::AtomSerializer.new.unfavourite_salmon(favourite))
  end
end
