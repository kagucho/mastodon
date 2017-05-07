# frozen_string_literal: true

module Admin
  class DomainBlocksController < BaseController
    before_action :set_domain_block, only: [:show, :destroy]

    def index
      @domain_blocks = DomainBlock.page(params[:page])
      set_csp
    end

    def new
      @domain_block = DomainBlock.new
      set_csp
    end

    def create
      @domain_block = DomainBlock.new(resource_params)

      if @domain_block.save
        DomainBlockWorker.perform_async(@domain_block.id)
        redirect_to admin_domain_blocks_path, notice: I18n.t('admin.domain_blocks.created_msg')
      else
        set_csp
        render :new
      end
    end

    def show
      set_csp
    end

    def destroy
      UnblockDomainService.new.call(@domain_block, retroactive_unblock?)
      redirect_to admin_domain_blocks_path, notice: I18n.t('admin.domain_blocks.destroyed_msg')
    end

    private

    def set_domain_block
      @domain_block = DomainBlock.find(params[:id])
    end

    def resource_params
      params.require(:domain_block).permit(:domain, :severity, :reject_media, :retroactive)
    end

    def retroactive_unblock?
      ActiveRecord::Type.lookup(:boolean).cast(resource_params[:retroactive])
    end

    def set_csp
      response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
    end
  end
end
