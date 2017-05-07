# frozen_string_literal: true

class AccountsController < ApplicationController
  include AccountControllerConcern

  def show
    respond_to do |format|
      format.html do
        @statuses = @account.statuses.permitted_for(@account, current_account).paginate_by_max_id(20, params[:max_id], params[:since_id])
        @statuses = cache_collection(@statuses, Status)

        response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET} #{ContentSecurityPolicy.digest(@card_style)} #{StreamEntriesHelper::CSP_STYLE_SRC}"
      end

      format.atom do
        @entries = @account.stream_entries.where(hidden: false).with_includes.paginate_by_max_id(20, params[:max_id], params[:since_id])
        render xml: AtomSerializer.render(AtomSerializer.new.feed(@account, @entries.to_a))
      end

      format.activitystreams2
    end
  end

  private

  def set_account
    @account = Account.find_local!(params[:username])
  end
end
